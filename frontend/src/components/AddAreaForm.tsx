import React, { useState } from 'react';
import api from '../api/api';

function AddAreaForm() {
  const [name, setName] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);
  // const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrors([]);

    setIsLoading(true);

    try {
      const response = await api.post('/areas', {
        data: {
          name,
        },
      });

      // eslint-disable-next-line no-console
      console.log('New area created:', response.data);
      setName('');
    } catch (error: any) {
      // eslint-disable-next-line no-console
      console.error('Error creating area:', error);
      if (Array.isArray(error.response?.data?.errors) && error.response.data.errors.length > 0) {
        setErrors(error.response.data.errors);
        /*
        errors:
        - error:
            type: "validation.Errors"
            error:
              data:
                <field>: <error message>
          status: "Unprocessable Entity"
         */

        // eslint-disable-next-line no-console
        console.log(error.response);
        // eslint-disable-next-line no-console
        console.log(error.response.data);

        error.response.data.errors.forEach((e: any) => {
          if (e.type === 'validation.Errors' && e.error?.data) {
            if (typeof e.error?.data === 'object' && Object.keys(e.error.data).length > 0) {
              // eslint-disable-next-line no-console
              console.log(e.error.data);
            }
          }
        });

        const errorMessage = error.response.data.errors[0].error.error.data.location_id;
        setErrors([`location_id: ${errorMessage}`]);
      } else {
        setErrors(['Error creating area. Please try again.']);
      }
    }

    setIsLoading(false);
  };

  return (
    <div>
      <h2>Add New Area</h2>
      {errors && <div>{errors}</div>}
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          placeholder="Area Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <button type="submit" disabled={isLoading}>
          {isLoading ? 'Adding...' : 'Add Area'}
        </button>
      </form>
    </div>
  );
}

export default AddAreaForm;
