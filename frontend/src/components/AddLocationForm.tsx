import React, { useState } from 'react';
import api from '../api/api';

// interface AddLocationFormProps {
//   onLocationAdded: () => void;
// }

function AddLocationForm({ onLocationAdded }:any) {
  const [name, setName] = useState('');
  const [address, setAddress] = useState('');
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    try {
      /* const response = */ await api.post('/locations', {
        data: {
          name,
          address,
        },
      });

      setName('');
      setAddress('');
      setError('');
      onLocationAdded(); // Invoke the callback function to trigger re-fetching of locations
    } catch (error: any) {
      if (error.response && error.response.data && error.response.data.errors) {
        const errorMessage = error.response.data.errors[0].error.error.data.name; // Modify the error message based on your API response
        setError(errorMessage);
      } else {
        setError('Error creating location');
      }
    }
  };

  return (
    <div>
      <h2>Add Location</h2>
      <form onSubmit={handleSubmit}>
        <div>
          {/* eslint-disable-next-line jsx-a11y/label-has-associated-control */}
          <label htmlFor="name">Name:</label>
          <input type="text" id="name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div>
          {/* eslint-disable-next-line jsx-a11y/label-has-associated-control */}
          <label htmlFor="address">Address:</label>
          <input type="text" id="address" value={address} onChange={(e) => setAddress(e.target.value)} />
        </div>
        <button type="submit">Add Location</button>
        {error && (
        <div>
          Error:
          {error}
        </div>
        )}
      </form>
    </div>
  );
}

export default AddLocationForm;
